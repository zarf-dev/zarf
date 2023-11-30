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
	checkforUnpinnedRepos(&validator)
	checkForUnpinnedImages(&validator)
	checkForUnpinnedFiles(&validator)

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

func imageIsPinned(image string) bool {
	if strings.HasSuffix(image, ":latest") {
		return false
	}
	// Pinned with image digest
	if strings.Contains(image, "@") {
		return true
	}
	splitImage := strings.Split(image, ":")
	if len(splitImage) > 1 {
		// In this case we either have a port and a "/" afterwards for the path to the image
		// Or we have a tag which cannot have the / character in it
		if !strings.Contains(splitImage[len(splitImage)-1], "/") {
			return true
		}
	}
	return false
}

func repoIsPinned(repo string) bool {
	// Pinned github and dev.azure.com repos will have @
	// Pinned gitlab repos will have /-/
	return (strings.Contains(repo, "@") || strings.Contains(repo, "/-/"))
}

func checkforUnpinnedRepos(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		for j, repo := range component.Repos {
			if !repoIsPinned(repo) {
				validator.addWarning(fmt.Sprintf(".components.[%d].repos.[%d]: Unpinned repository", i, j))
			}
		}
	}
}

func checkForUnpinnedImages(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		for j, image := range component.Images {
			if !imageIsPinned(image) {
				validator.addWarning(fmt.Sprintf(".components.[%d].images.[%d]: Unpinned image", i, j))
			}
		}
	}
}

func checkForUnpinnedFiles(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		for j, file := range component.Files {
			if file.Shasum == "" {
				validator.addWarning(fmt.Sprintf(".components.[%d].files.[%d]: Unpinned file", i, j))
			}
		}
	}
}

func checkForVarInComponentImport(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
			validator.addWarning(fmt.Sprintf(".components.[%d].import.path: Will not resolve ZARF_PKG_TMPL_* variables", i))
		}
		if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
			validator.addWarning(fmt.Sprintf(".components.[%d].import.url: Will not resolve ZARF_PKG_TMPL_* variables", i))
		}
	}

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
				"%s: %s", makeFieldPathYqCompat(desc.Field()), desc.Description())
			validator.addError(err)
		}
	}

	return err
}
