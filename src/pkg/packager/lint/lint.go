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

	lintComposableComponents(&validator, &createOpts)

	if validator.jsonSchema, err = getSchemaFile(); err != nil {
		return nil, err
	}
	pkgKey := packageKey{name: validator.typedZarfPackage.Metadata.Name, path: createOpts.BaseDir}
	if err = validateSchema(&validator, pkgKey); err != nil {
		return nil, err
	}

	return &validator, nil
}

func lintComposableComponents(validator *Validator, createOpts *types.ZarfCreateOptions) {
	for i, component := range validator.typedZarfPackage.Components {
		arch := config.GetArch(validator.typedZarfPackage.Metadata.Architecture)

		if !composer.CompatibleComponent(component, arch, createOpts.Flavor) {
			continue
		}

		chain, err := composer.NewImportChain(component, i, validator.typedZarfPackage.Metadata.Name, arch, createOpts.Flavor)
		baseComponent := chain.Head()
		var badImportYqPath string
		if baseComponent != nil {
			if baseComponent.Import.URL != "" {
				badImportYqPath = fmt.Sprintf(".components.[%d].import.url", i)
			}
			if baseComponent.Import.Path != "" {
				badImportYqPath = fmt.Sprintf(".components.[%d].import.path", i)
			}
		}
		if err != nil {
			validator.addError(validatorMessage{
				description: err.Error(),
				packageKey:  packageKey{name: validator.typedZarfPackage.Metadata.Name},
				yqPath:      badImportYqPath,
			})
		}

		node := baseComponent
		for node != nil {
			fileOrOciPath := createOpts.BaseDir
			if node.Prev() != nil {
				if node.Prev().Import.URL != "" {
					fileOrOciPath = node.Prev().Import.URL
				} else if node.GetRelativeToHead() != "" {
					fileOrOciPath = filepath.Join(createOpts.BaseDir, node.GetRelativeToHead())
				}
			}
			pkgKey := packageKey{path: fileOrOciPath, name: node.GetOriginalPackageName()}
			checkForVarInComponentImport(validator, node.GetIndex(), node.ZarfComponent, pkgKey)
			fillComponentTemplate(validator, node, createOpts, pkgKey)
			lintComponent(validator, node.GetIndex(), node.ZarfComponent, pkgKey)
			node = node.Next()
		}
	}
}

func fillComponentTemplate(validator *Validator, node *composer.Node, createOpts *types.ZarfCreateOptions, pkgKey packageKey) {

	err := packager.ReloadComponentTemplate(&node.ZarfComponent)
	if err != nil {
		validator.addWarning(validatorMessage{
			description: err.Error(),
			packageKey:  pkgKey,
		})
	}
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) {
		yamlTemplates, err := utils.FindYamlTemplates(node, templatePrefix, "###")
		if err != nil {
			validator.addWarning(validatorMessage{
				description: err.Error(),
				packageKey:  pkgKey,
			})
		}

		for key := range yamlTemplates {
			if deprecated {
				validator.addWarning(validatorMessage{
					description: fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					packageKey:  pkgKey,
				})
			}
			_, present := createOpts.SetVariables[key]
			if !present {
				validator.addWarning(validatorMessage{
					description: lang.UnsetVarLintWarning,
					packageKey:  pkgKey,
				})
			}
		}
		for key, value := range createOpts.SetVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
	}

	setVarsAndWarn(types.ZarfPackageTemplatePrefix, false)

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	setVarsAndWarn(types.ZarfPackageVariablePrefix, true)

	utils.ReloadYamlTemplate(node, templateMap)
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

func lintComponent(validator *Validator, index int, component types.ZarfComponent, pkgKey packageKey) {
	checkForUnpinnedRepos(validator, index, component, pkgKey)
	checkForUnpinnedImages(validator, index, component, pkgKey)
	checkForUnpinnedFiles(validator, index, component, pkgKey)
}

func checkForUnpinnedRepos(validator *Validator, index int, component types.ZarfComponent, pkgKey packageKey) {
	for j, repo := range component.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", index, j)
		if !isPinnedRepo(repo) {
			validator.addWarning(validatorMessage{
				yqPath:      repoYqPath,
				packageKey:  pkgKey,
				description: "Unpinned repository",
				item:        repo,
			})
		}
	}
}

func checkForUnpinnedImages(validator *Validator, index int, component types.ZarfComponent, pkgKey packageKey) {
	for j, image := range component.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", index, j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			validator.addError(validatorMessage{
				yqPath:      imageYqPath,
				packageKey:  pkgKey,
				description: "Invalid image format",
				item:        image,
			})
			continue
		}
		if !pinnedImage {
			validator.addWarning(validatorMessage{
				yqPath:      imageYqPath,
				packageKey:  pkgKey,
				description: "Image not pinned with digest",
				item:        image,
			})
		}
	}
}

func checkForUnpinnedFiles(validator *Validator, index int, component types.ZarfComponent, pkgKey packageKey) {
	for j, file := range component.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", index, j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			validator.addWarning(validatorMessage{
				yqPath:      fileYqPath,
				packageKey:  pkgKey,
				description: "No shasum for remote file",
				item:        file.Source,
			})
		}
	}
}

func checkForVarInComponentImport(validator *Validator, index int, component types.ZarfComponent, pkgKey packageKey) {
	if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(validatorMessage{
			yqPath:      fmt.Sprintf(".components.[%d].import.path", index),
			packageKey:  pkgKey,
			description: "Zarf does not evaluate variables at component.x.import.path",
			item:        component.Import.Path,
		})
	}
	if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(validatorMessage{
			yqPath:      fmt.Sprintf(".components.[%d].import.url", index),
			packageKey:  pkgKey,
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

func validateSchema(validator *Validator, pkgKey packageKey) error {
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
				packageKey:  pkgKey,
			})
		}
	}

	return err
}
