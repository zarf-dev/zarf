// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
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

// Validate validates a zarf file against the zarf schema, returns *validator with warnings or errors if they exist
// along with an error if the validation itself failed
func Validate(cfg *types.PackagerConfig) (*Validator, error) {
	validator := Validator{}
	var err error

	if err := utils.ReadYaml(filepath.Join(cfg.CreateOpts.BaseDir, layout.ZarfYAML), &validator.typedZarfPackage); err != nil {
		return nil, err
	}

	if err := utils.ReadYaml(filepath.Join(cfg.CreateOpts.BaseDir, layout.ZarfYAML), &validator.untypedZarfPackage); err != nil {
		return nil, err
	}

	if err := os.Chdir(cfg.CreateOpts.BaseDir); err != nil {
		return nil, fmt.Errorf("unable to access directory '%s': %w", cfg.CreateOpts.BaseDir, err)
	}

	validator.baseDir = cfg.CreateOpts.BaseDir

	values, err := template.Generate(cfg)
	// If this breaks what do we want to do
	if err != nil {
		panic("fix this")
	}
	// Make list of custom variables
	templateMap := values.GetCustomVariables()
	for key := range templateMap {
		validator.unusedVariables = append(validator.unusedVariables, key)
	}

	lintComponents(&validator, cfg)

	if validator.jsonSchema, err = getSchemaFile(); err != nil {
		return nil, err
	}

	if err = validateSchema(&validator); err != nil {
		return nil, err
	}

	return &validator, nil
}

func lintComponents(validator *Validator, cfg *types.PackagerConfig) {
	for i, component := range validator.typedZarfPackage.Components {
		arch := config.GetArch(validator.typedZarfPackage.Metadata.Architecture)

		if !composer.CompatibleComponent(component, arch, cfg.CreateOpts.Flavor) {
			continue
		}

		chain, err := composer.NewImportChain(component, i, validator.typedZarfPackage.Metadata.Name, arch, cfg.CreateOpts.Flavor)
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
				description:    err.Error(),
				packageRelPath: ".",
				packageName:    validator.typedZarfPackage.Metadata.Name,
				yqPath:         badImportYqPath,
			})
		}

		node := baseComponent
		for node != nil {
			checkForVarInComponentImport(validator, node)
			fillComponentTemplate(validator, node, &cfg.CreateOpts)
			lintComponent(validator, node)
			checkForUnusedVariables(validator, cfg, node)
			node = node.Next()
		}
	}
}

func reloadComponentTemplate(component *types.ZarfComponent) error {
	mappings := map[string]string{}
	mappings[types.ZarfComponentName] = component.Name
	err := utils.ReloadYamlTemplate(component, mappings)
	if err != nil {
		return err
	}
	return nil
}

func fillComponentTemplate(validator *Validator, node *composer.Node, createOpts *types.ZarfCreateOptions) {

	err := reloadComponentTemplate(&node.ZarfComponent)
	if err != nil {
		validator.addWarning(validatorMessage{
			description:    err.Error(),
			packageRelPath: node.ImportLocation(),
			packageName:    node.GetOriginalPackageName(),
		})
	}
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) {
		yamlTemplates, err := utils.FindYamlTemplates(node, templatePrefix, "###")
		if err != nil {
			validator.addWarning(validatorMessage{
				description:    err.Error(),
				packageRelPath: node.ImportLocation(),
				packageName:    node.GetOriginalPackageName(),
			})
		}

		for key := range yamlTemplates {
			if deprecated {
				validator.addWarning(validatorMessage{
					description:    fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					packageRelPath: node.ImportLocation(),
					packageName:    node.GetOriginalPackageName(),
				})
			}
			_, present := createOpts.SetVariables[key]
			if !present {
				validator.addWarning(validatorMessage{
					description:    lang.UnsetVarLintWarning,
					packageRelPath: node.ImportLocation(),
					packageName:    node.GetOriginalPackageName(),
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

func lintComponent(validator *Validator, node *composer.Node) {
	checkForUnpinnedRepos(validator, node)
	checkForUnpinnedImages(validator, node)
	checkForUnpinnedFiles(validator, node)
}

func checkForUnpinnedRepos(validator *Validator, node *composer.Node) {
	for j, repo := range node.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", node.GetIndex(), j)
		if !isPinnedRepo(repo) {
			validator.addWarning(validatorMessage{
				yqPath:         repoYqPath,
				packageRelPath: node.ImportLocation(),
				packageName:    node.GetOriginalPackageName(),
				description:    "Unpinned repository",
				item:           repo,
			})
		}
	}
}

func checkForUnpinnedImages(validator *Validator, node *composer.Node) {
	for j, image := range node.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", node.GetIndex(), j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			validator.addError(validatorMessage{
				yqPath:         imageYqPath,
				packageRelPath: node.ImportLocation(),
				packageName:    node.GetOriginalPackageName(),
				description:    "Invalid image reference",
				item:           image,
			})
			continue
		}
		if !pinnedImage {
			validator.addWarning(validatorMessage{
				yqPath:         imageYqPath,
				packageRelPath: node.ImportLocation(),
				packageName:    node.GetOriginalPackageName(),
				description:    "Image not pinned with digest",
				item:           image,
			})
		}
	}
}

func checkForUnpinnedFiles(validator *Validator, node *composer.Node) {
	for j, file := range node.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", node.GetIndex(), j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			validator.addWarning(validatorMessage{
				yqPath:         fileYqPath,
				packageRelPath: node.ImportLocation(),
				packageName:    node.GetOriginalPackageName(),
				description:    "No shasum for remote file",
				item:           file.Source,
			})
		}
	}
}

func checkForVarInComponentImport(validator *Validator, node *composer.Node) {
	if strings.Contains(node.Import.Path, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(validatorMessage{
			yqPath:         fmt.Sprintf(".components.[%d].import.path", node.GetIndex()),
			packageRelPath: node.ImportLocation(),
			packageName:    node.GetOriginalPackageName(),
			description:    "Zarf does not evaluate variables at component.x.import.path",
			item:           node.Import.Path,
		})
	}
	if strings.Contains(node.Import.URL, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(validatorMessage{
			yqPath:         fmt.Sprintf(".components.[%d].import.url", node.GetIndex()),
			packageRelPath: node.ImportLocation(),
			packageName:    node.GetOriginalPackageName(),
			description:    "Zarf does not evaluate variables at component.x.import.url",
			item:           node.Import.URL,
		})
	}
}

func varUsedInFile(filepath string, variable string, deprecations map[string]string, templateRegex string) (bool, error) {
	textFile, err := os.Open(filepath)
	// TODO what is the behavior if we get to a file we can't open? Debug this message?
	if err != nil {
		return false, err
	}

	// This regex takes a line and parses the text before and after a discovered template: https://regex101.com/r/ilUxAz/1
	regexTemplateLine := regexp.MustCompile(fmt.Sprintf("(?P<preTemplate>.*?)(?P<template>%s)(?P<postTemplate>.*)", templateRegex))

	fileScanner := bufio.NewScanner(textFile)

	// Set the buffer to 1 MiB to handle long lines (i.e. base64 text in a secret)
	// 1 MiB is around the documented maximum size for secrets and configmaps
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	fileScanner.Buffer(buf, maxCapacity)

	// Set the scanner to split on new lines
	fileScanner.Split(bufio.ScanLines)

	text := ""

	for fileScanner.Scan() {
		line := fileScanner.Text()

		for {
			matches := regexTemplateLine.FindStringSubmatch(line)

			// No template left on this line so move on
			if len(matches) == 0 {
				text += fmt.Sprintln(line)
				break
			}

			preTemplate := matches[regexTemplateLine.SubexpIndex("preTemplate")]
			templateKey := matches[regexTemplateLine.SubexpIndex("template")]

			_, present := deprecations[templateKey]
			if present {
				message.Warnf("This Zarf Package uses a deprecated variable: '%s' changed to '%s'.  Please notify your package creator for an update.", templateKey, deprecations[templateKey])
			}

			template, present := mappings[templateKey]

			// Check if the template is nil (present), use the original templateKey if not (so that it is not replaced).
			value := templateKey
			if template != nil {
				value = template.Value

				// Check if the value is a file type and load the value contents from the file
				if template.Type == types.FileVariableType && value != "" {
					if isText, err := IsTextFile(value); err != nil || !isText {
						message.Warnf("Refusing to load a non-text file for templating %s", templateKey)
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					contents, err := os.ReadFile(value)
					if err != nil {
						message.Warnf("Unable to read file for templating - skipping: %s", err.Error())
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					value = string(contents)
				}

				// Check if the value is autoIndented and add the correct spacing
				if template.AutoIndent {
					indent := fmt.Sprintf("\n%s", strings.Repeat(" ", len(preTemplate)))
					value = strings.ReplaceAll(value, "\n", indent)
				}
			}

			// Add the processed text and continue processing the line
			text += fmt.Sprintf("%s%s", preTemplate, value)
			line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
		}
	}

	textFile.Close()

	return false, nil
}

// Potentially it is time to move the main function into packager
// this can have the package and get things with it
// Or I can keep moving things out of packager and make them more generic functions
func checkForUnusedVariables(validator *Validator, cfg *types.PackagerConfig, node *composer.Node) {
	// There are at least three different scenarios I need to cover
	// 1. The variables are in the actions of the zarf chart
	// 2. The variables are in a helm chart in the component
	// 3. The variables are in a file brough in by zarf
	// Initial idea is to go through each of these and as a variable is found, take it out of the list
	// At the end we warn that whatever is still in the list is unused.
	// We will also want to do this with both zarf const and zarf var
	// Where / how are constant variables set?

	//err := utils.ReplaceTextTemplate(path, templateMap, deprecations, "###ZARF_[A-Z0-9_]+###")

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
				yqPath:         makeFieldPathYqCompat(desc.Field()),
				description:    desc.Description(),
				packageRelPath: ".",
				packageName:    validator.typedZarfPackage.Metadata.Name,
			})
		}
	}

	return err
}
