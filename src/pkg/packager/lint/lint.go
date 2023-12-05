// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/packager/composer"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
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
func ValidateZarfSchema(createOpts types.ZarfCreateOptions) (*Validator, error) {
	validator := Validator{}
	var err error

	if err := utils.ReadYaml(filepath.Join(createOpts.BaseDir, layout.ZarfYAML), &validator.typedZarfPackage); err != nil {
		return nil, err
	}

	if err := utils.ReadYaml(filepath.Join(createOpts.BaseDir, layout.ZarfYAML), &validator.untypedZarfPackage); err != nil {
		return nil, err
	}

	if err := fillActiveTemplate(&validator, createOpts); err != nil {
		return nil, err
	}

	lintComponents(&validator)

	if err := os.Chdir(createOpts.BaseDir); err != nil {
		return nil, fmt.Errorf("unable to access directory '%s': %w", createOpts.BaseDir, err)
	}

	if err := ValidateComposableComponenets(&validator, createOpts); err != nil {
		return nil, err
	}

	if validator.jsonSchema, err = getSchemaFile(); err != nil {
		return nil, err
	}

	if err = validateSchema(&validator); err != nil {
		return nil, err
	}

	return &validator, nil
}

func ValidateComposableComponenets(validator *Validator, createOpts types.ZarfCreateOptions) error {
	for i, component := range validator.typedZarfPackage.Components {
		//TODO allow this to be a CLI option
		arch := config.GetArch(validator.typedZarfPackage.Metadata.Architecture)

		// filter by architecture
		if !composer.CompatibleComponent(component, arch, createOpts.Flavor) {
			continue
		}

		// if a match was found, strip flavor and architecture to reduce bloat in the package definition
		component.Only.Cluster.Architecture = ""
		component.Only.Flavor = ""

		// build the import chain
		chain, err := composer.NewImportChain(component, i, arch, createOpts.Flavor)
		if err != nil {
			return err
		}
		originalPackage := validator.typedZarfPackage
		// Skipping initial component
		node := chain.Head.Next
		for node != nil {
			validator.typedZarfPackage.Components = []types.ZarfComponent{node.ZarfComponent}
			fillActiveTemplate(validator, createOpts)
			lintComponent(validator, node.Index, validator.typedZarfPackage.Components[0], fmt.Sprintf(" %s", node.RelativeToHead))
			validator.typedZarfPackage = originalPackage
			node = node.Next
		}

	}
	return nil
}

func fillActiveTemplate(validator *Validator, createOpts types.ZarfCreateOptions) error {
	templateMap := map[string]string{}
	unsetVarWarning := false

	promptAndSetTemplate := func(templatePrefix string, deprecated bool) error {
		yamlTemplates, err := utils.FindYamlTemplates(validator.typedZarfPackage, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				validator.warnings = append(validator.warnings, fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key))
			}
			_, present := createOpts.SetVariables[key]
			if !present {
				unsetVarWarning = true
			}
		}

		for key, value := range createOpts.SetVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
		return nil
	}

	// update the component templates on the package
	err := packager.FindComponentTemplatesAndReload(&validator.typedZarfPackage)
	if err != nil {
		return err
	}

	if err := promptAndSetTemplate(types.ZarfPackageTemplatePrefix, false); err != nil {
		return err
	}

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := promptAndSetTemplate(types.ZarfPackageVariablePrefix, true); err != nil {
		return err
	}

	// Add special variable for the current package architecture
	templateMap[types.ZarfPackageArch] = config.GetArch(validator.typedZarfPackage.Metadata.Architecture, validator.typedZarfPackage.Build.Architecture)

	if unsetVarWarning {
		validator.warnings = append([]string{"There are variables that are unset and won't be evaluated during lint"}, validator.warnings...)
	}

	return utils.ReloadYamlTemplate(&validator.typedZarfPackage, templateMap)
}

func isPinnedImage(image string) (bool, error) {
	transformedImage, err := transform.ParseImageRef(image)
	if err != nil {
		if strings.Contains(image, types.ZarfPackageTemplatePrefix) ||
			strings.Contains(image, types.ZarfPackageVariablePrefix) {
			return true, nil
		}
		return false, err
	}
	return (transformedImage.Digest != ""), err
}

func isPinnedRepo(repo string) bool {
	// Pinned github and dev.azure.com repos will have @
	// Pinned gitlab repos will have /-/
	return (strings.Contains(repo, "@") || strings.Contains(repo, "/-/"))
}

// Feels like validator may have too much with both the zarf package and the warnings
func lintComponents(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		lintComponent(validator, i, component, "")
	}
}

func lintComponent(validator *Validator, index int, component types.ZarfComponent, path string) {
	checkforUnpinnedRepos(validator, index, component, path)
	checkForUnpinnedImages(validator, index, component, path)
	checkForUnpinnedFiles(validator, index, component, path)
	checkForVarInComponentImport(validator, index, component, path)
}

func checkforUnpinnedRepos(validator *Validator, index int, component types.ZarfComponent, path string) {
	for j, repo := range component.Repos {
		if !isPinnedRepo(repo) {
			validator.addWarning(fmt.Sprintf(".components.[%d].repos.[%d]%s: Unpinned repository", index, j, path))
		}
	}
}

func checkForUnpinnedImages(validator *Validator, index int, component types.ZarfComponent, path string) {
	for j, image := range component.Images {
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			validator.addError(fmt.Errorf(".components.[%d].images.[%d]%s: Invalid image format", index, j, path))
			continue
		}
		if !pinnedImage {
			validator.addWarning(fmt.Sprintf(".components.[%d].images.[%d]%s: Unpinned image", index, j, path))
		}
	}
}

func checkForUnpinnedFiles(validator *Validator, index int, component types.ZarfComponent, path string) {
	for j, file := range component.Files {
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			validator.addWarning(fmt.Sprintf(".components.[%d].files.[%d]%s: Unpinned file", index, j, path))
		}
	}
}

func checkForVarInComponentImport(validator *Validator, index int, component types.ZarfComponent, path string) {
	if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(fmt.Sprintf(".components.[%d].import.path%s: Will not resolve ZARF_PKG_TMPL_* variables", index, path))
	}
	if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(fmt.Sprintf(".components.[%d].import.url%s: Will not resolve ZARF_PKG_TMPL_* variables", index, path))
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
