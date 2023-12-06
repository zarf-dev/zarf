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

	if err := os.Chdir(createOpts.BaseDir); err != nil {
		return nil, fmt.Errorf("unable to access directory '%s': %w", createOpts.BaseDir, err)
	}

	if err := LintComposableComponenets(&validator, createOpts); err != nil {
		return nil, err
	}

	preVarEvalLintComponents(&validator)

	if err := fillActiveTemplate(&validator, createOpts); err != nil {
		return nil, err
	}

	lintComponents(&validator)

	if validator.jsonSchema, err = getSchemaFile(); err != nil {
		return nil, err
	}

	if err = validateSchema(&validator); err != nil {
		return nil, err
	}

	return &validator, nil
}

func LintComposableComponenets(validator *Validator, createOpts types.ZarfCreateOptions) error {
	for i, component := range validator.typedZarfPackage.Components {
		arch := config.GetArch(validator.typedZarfPackage.Metadata.Architecture)

		if !composer.CompatibleComponent(component, arch, createOpts.Flavor) {
			continue
		}

		// This is done in composer, should I just delete this or move it into newImportChain
		// component.Only.Cluster.Architecture = ""
		// component.Only.Flavor = ""

		chain, err := composer.NewImportChain(component, i, arch, createOpts.Flavor)
		if err != nil {
			return err
		}

		// Skipping initial component since it will be linted the usual way
		node := chain.Head.Next()
		path := chain.GetRemoteName()
		for node != nil {
			if path == "" {
				path = node.GetRelativeToHead()
			}
			checkForVarInComponentImport(validator, node.GetIndex(), node.ZarfComponent, path)
			fillComponentTemplate(validator, &node.ZarfComponent, createOpts)
			lintComponent(validator, node.GetIndex(), node.ZarfComponent, path)
			node = node.Next()
		}
	}
	return nil
}

func fillComponentTemplate(validator *Validator, component *types.ZarfComponent, createOpts types.ZarfCreateOptions) error {
	// update the component templates on the package
	err := packager.ReloadComponentTemplate(component)
	if err != nil {
		return err
	}
	return fillYamlTemplate(validator, component, createOpts)
}

// Look into breaking apart this function to allow for passing in a component
// Look into returning warnings so I don't have to pass in validator
// Look into removing package from validator
func fillActiveTemplate(validator *Validator, createOpts types.ZarfCreateOptions) error {

	// update the component templates on the package
	// TODO add test to make sure this is covered still
	err := packager.FindComponentTemplatesAndReload(&validator.typedZarfPackage)
	if err != nil {
		return err
	}
	//Does typed zarf package need to be addressed here or is it already addresssed
	return fillYamlTemplate(validator, &validator.typedZarfPackage, createOpts)
}

func fillYamlTemplate(validator *Validator, yamlObj any, createOpts types.ZarfCreateOptions) error {
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) error {
		yamlTemplates, err := utils.FindYamlTemplates(yamlObj, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				validator.addWarning(ValidatorMessage{
					description: fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
				})
			}
			_, present := createOpts.SetVariables[key]
			if !present && !validator.hasUnSetVarWarning {
				validator.warnings = append([]ValidatorMessage{{
					description: "There are variables that are unset and won't be evaluated during lint",
				}}, validator.warnings...)
				validator.hasUnSetVarWarning = true
			}
		}

		for key, value := range createOpts.SetVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
		return nil
	}

	if err := setVarsAndWarn(types.ZarfPackageTemplatePrefix, false); err != nil {
		return err
	}

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := setVarsAndWarn(types.ZarfPackageVariablePrefix, true); err != nil {
		return err
	}

	return utils.ReloadYamlTemplate(yamlObj, templateMap)
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
	return (strings.Contains(repo, "@"))
}

// Feels like validator may have too much with both the zarf package and the warnings
func lintComponents(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		lintComponent(validator, i, component, "")
	}
}

func lintComponent(validator *Validator, index int, component types.ZarfComponent, path string) {
	checkForUnpinnedRepos(validator, index, component, path)
	checkForUnpinnedImages(validator, index, component, path)
	checkForUnpinnedFiles(validator, index, component, path)
}

func checkForUnpinnedRepos(validator *Validator, index int, component types.ZarfComponent, path string) {
	for j, repo := range component.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", index, j)
		if !isPinnedRepo(repo) {
			validator.addWarning(ValidatorMessage{
				yqPath:      repoYqPath,
				filePath:    path,
				description: "Unpinned repository",
				item:        repo,
			})
		}
	}
}

func checkForUnpinnedImages(validator *Validator, index int, component types.ZarfComponent, path string) {
	for j, image := range component.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", index, j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			validator.addError(ValidatorMessage{
				yqPath:      imageYqPath,
				filePath:    path,
				description: "Invalid image format",
				item:        image,
			})
			continue
		}
		if !pinnedImage {
			validator.addWarning(ValidatorMessage{
				yqPath:      imageYqPath,
				filePath:    path,
				description: "Image not pinned with digest",
				item:        image,
			})
		}
	}
}

func checkForUnpinnedFiles(validator *Validator, index int, component types.ZarfComponent, path string) {
	for j, file := range component.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", index, j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			validator.addWarning(ValidatorMessage{
				yqPath:      fileYqPath,
				filePath:    path,
				description: "No shasum for remote file",
				item:        file.Source,
			})
		}
	}
}

func preVarEvalLintComponents(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		checkForVarInComponentImport(validator, i, component, "")
	}
}

func checkForVarInComponentImport(validator *Validator, index int, component types.ZarfComponent, path string) {
	if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(ValidatorMessage{
			yqPath:      fmt.Sprintf(".components.[%d].import.path", index),
			filePath:    path,
			description: "Zarf does not evaluate variables at component.x.import.path",
			item:        component.Import.Path,
		})
	}
	if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(ValidatorMessage{
			yqPath:      fmt.Sprintf(".components.[%d].import.url", index),
			filePath:    path,
			description: "Zarf does not evaluate variables at component.x.import.url",
			item:        component.Import.URL,
		})
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
			validator.addError(ValidatorMessage{
				yqPath:      makeFieldPathYqCompat(desc.Field()),
				description: desc.Description(),
			})
		}
	}

	return err
}
