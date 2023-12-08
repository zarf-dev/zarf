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

	lintComposableComponenets(&validator, createOpts)

	lintUnEvaledVariables(&validator)

	fillActiveTemplate(&validator, createOpts)

	lintComponents(&validator)

	if validator.jsonSchema, err = getSchemaFile(); err != nil {
		return nil, err
	}

	if err = validateSchema(&validator); err != nil {
		return nil, err
	}

	return &validator, nil
}

func lintComposableComponenets(validator *Validator, createOpts types.ZarfCreateOptions) {
	for i, component := range validator.typedZarfPackage.Components {
		arch := config.GetArch(validator.typedZarfPackage.Metadata.Architecture)

		if !composer.CompatibleComponent(component, arch, createOpts.Flavor) {
			continue
		}

		chain, err := composer.NewImportChain(component, i, validator.typedZarfPackage.Metadata.Name, arch, createOpts.Flavor)
		if err != nil {
			validator.addError(validatorMessage{
				description: err.Error(),
				packageName: validator.typedZarfPackage.Metadata.Name,
			})
			continue
		}

		path := chain.Head().Import.URL
		// Skipping initial component since it will be linted the usual way
		node := chain.Head().Next()
		for node != nil {
			if path == "" {
				path = node.GetRelativeToHead()
			}
			checkForVarInComponentImport(validator, node.GetIndex(), node.ZarfComponent, path, node.GetOriginalPackageName())
			fillComponentTemplate(validator, node, createOpts)
			lintComponent(validator, node.GetIndex(), node.ZarfComponent, path, node.GetOriginalPackageName())
			node = node.Next()
		}
	}
}

func fillComponentTemplate(validator *Validator, node *composer.Node, createOpts types.ZarfCreateOptions) {

	err := packager.ReloadComponentTemplate(&node.ZarfComponent)
	if err != nil {
		validator.addWarning(validatorMessage{
			description: fmt.Sprintf("unable to find components %s", err),
			filePath:    node.GetRelativeToHead(),
			packageName: node.GetOriginalPackageName(),
		})
	}
	fillYamlTemplate(validator, node, createOpts, node.GetOriginalPackageName())
}

func fillActiveTemplate(validator *Validator, createOpts types.ZarfCreateOptions) {

	err := packager.FindComponentTemplatesAndReload(&validator.typedZarfPackage)
	if err != nil {
		validator.addWarning(validatorMessage{
			description: fmt.Sprintf("unable to find components %s", err),
			packageName: validator.typedZarfPackage.Metadata.Name,
		})
	}

	fillYamlTemplate(validator, &validator.typedZarfPackage, createOpts, validator.typedZarfPackage.Metadata.Name)
}

func fillYamlTemplate(validator *Validator, yamlObj any, createOpts types.ZarfCreateOptions, pkgName string) {
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) {
		yamlTemplates, err := utils.FindYamlTemplates(yamlObj, templatePrefix, "###")
		if err != nil {
			validator.addWarning(validatorMessage{
				description: fmt.Sprintf("unable to find variables %s", err),
				packageName: pkgName,
			})
		}

		for key := range yamlTemplates {
			if deprecated {
				validator.addWarning(validatorMessage{
					description: fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					packageName: pkgName,
				})
			}
			_, present := createOpts.SetVariables[key]
			if !present && !validator.hasUnSetVarWarning {
				validator.findings = append([]validatorMessage{{
					description: "There are variables that are unset and won't be evaluated during lint",
					packageName: pkgName,
				}}, validator.findings...)
				validator.hasUnSetVarWarning = true
			}
		}

		for key, value := range createOpts.SetVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
	}

	setVarsAndWarn(types.ZarfPackageTemplatePrefix, false)

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	setVarsAndWarn(types.ZarfPackageVariablePrefix, true)

	utils.ReloadYamlTemplate(yamlObj, templateMap)
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

func lintComponents(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		lintComponent(validator, i, component, "", validator.typedZarfPackage.Metadata.Name)
	}
}

func lintComponent(validator *Validator, index int, component types.ZarfComponent, path string, packageName string) {
	checkForUnpinnedRepos(validator, index, component, path, packageName)
	checkForUnpinnedImages(validator, index, component, path, packageName)
	checkForUnpinnedFiles(validator, index, component, path, packageName)
}

func checkForUnpinnedRepos(validator *Validator, index int, component types.ZarfComponent, path string, pkgName string) {
	for j, repo := range component.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", index, j)
		if !isPinnedRepo(repo) {
			validator.addWarning(validatorMessage{
				yqPath:      repoYqPath,
				filePath:    path,
				description: "Unpinned repository",
				item:        repo,
				packageName: pkgName,
			})
		}
	}
}

func checkForUnpinnedImages(validator *Validator, index int, component types.ZarfComponent, path string, pkgName string) {
	for j, image := range component.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", index, j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			validator.addError(validatorMessage{
				yqPath:      imageYqPath,
				filePath:    path,
				description: "Invalid image format",
				item:        image,
				packageName: pkgName,
			})
			continue
		}
		if !pinnedImage {
			validator.addWarning(validatorMessage{
				yqPath:      imageYqPath,
				filePath:    path,
				description: "Image not pinned with digest",
				item:        image,
				packageName: pkgName,
			})
		}
	}
}

func checkForUnpinnedFiles(validator *Validator, index int,
	component types.ZarfComponent, path string, pkgName string) {
	for j, file := range component.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", index, j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			validator.addWarning(validatorMessage{
				yqPath:      fileYqPath,
				filePath:    path,
				description: "No shasum for remote file",
				item:        file.Source,
				packageName: pkgName,
			})
		}
	}
}

func lintUnEvaledVariables(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		checkForVarInComponentImport(validator, i, component, "", validator.typedZarfPackage.Metadata.Name)
	}
}

func checkForVarInComponentImport(validator *Validator, index int, component types.ZarfComponent, path string, pkgName string) {
	if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(validatorMessage{
			yqPath:      fmt.Sprintf(".components.[%d].import.path", index),
			filePath:    path,
			description: "Zarf does not evaluate variables at component.x.import.path",
			item:        component.Import.Path,
			packageName: pkgName,
		})
	}
	if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(validatorMessage{
			yqPath:      fmt.Sprintf(".components.[%d].import.url", index),
			filePath:    path,
			description: "Zarf does not evaluate variables at component.x.import.url",
			item:        component.Import.URL,
			packageName: pkgName,
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
			validator.addError(validatorMessage{
				yqPath:      makeFieldPathYqCompat(desc.Field()),
				description: desc.Description(),
				packageName: validator.typedZarfPackage.Metadata.Name,
			})
		}
	}

	return err
}
