// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"embed"
	"fmt"
	"regexp"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/xeipuuv/gojsonschema"
)

// ZarfSchema is exported so main.go can embed the schema file
var ZarfSchema embed.FS

func getSchemaFile() ([]byte, error) {
	return ZarfSchema.ReadFile("zarf.schema.json")
}

// Validate validates a zarf file against the zarf schema, returns *validator with warnings or errors if they exist
// along with an error if the validation itself failed
func Validate(_ types.ZarfCreateOptions) (*Validator, error) {
	validator := Validator{}

	// if err := utils.ReadYaml(filepath.Join(createOpts.BaseDir, layout.ZarfYAML), &validator.typedZarfPackage); err != nil {
	// 	return nil, err
	// }

	// if err := utils.ReadYaml(filepath.Join(createOpts.BaseDir, layout.ZarfYAML), &validator.untypedZarfPackage); err != nil {
	// 	return nil, err
	// }

	// if err := os.Chdir(createOpts.BaseDir); err != nil {
	// 	return nil, fmt.Errorf("unable to access directory '%s': %w", createOpts.BaseDir, err)
	// }

	// validator.baseDir = createOpts.BaseDir

	// // lintComponents(&validator, &createOpts)

	// if validator.jsonSchema, err = getSchemaFile(); err != nil {
	// 	return nil, err
	// }

	// if err = validateSchema(&validator); err != nil {
	// 	return nil, err
	// }

	return &validator, nil
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
			validator.addError(ValidatorMessage{
				YqPath:         makeFieldPathYqCompat(desc.Field()),
				Description:    desc.Description(),
				PackageRelPath: ".",
				PackageName:    validator.typedZarfPackage.Metadata.Name,
			})
		}
	}

	return err
}
