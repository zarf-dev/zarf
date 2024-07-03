// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/composer"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/xeipuuv/gojsonschema"
)

// ZarfSchema is exported so main.go can embed the schema file
var ZarfSchema fs.ReadFileFS

// Validate the given Zarf package. The Zarf package should not already be composed when sent to this function.
func Validate(ctx context.Context, pkg types.ZarfPackage, setVariables map[string]string, flavor string) ([]types.PackageFinding, error) {
	var findings []types.PackageFinding
	compFindings, err := lintComponents(ctx, pkg, setVariables, flavor)
	if err != nil {
		return nil, err
	}
	findings = append(findings, compFindings...)

	jsonSchema, err := ZarfSchema.ReadFile("zarf.schema.json")
	if err != nil {
		return nil, err
	}

	var untypedZarfPackage interface{}
	if err := utils.ReadYaml(layout.ZarfYAML, &untypedZarfPackage); err != nil {
		return nil, err
	}

	schemaFindings, err := validateSchema(jsonSchema, untypedZarfPackage)
	if err != nil {
		return nil, err
	}
	findings = append(findings, schemaFindings...)

	return findings, nil
}

func lintComponents(ctx context.Context, pkg types.ZarfPackage, setVariables map[string]string, flavor string) ([]types.PackageFinding, error) {
	var findings []types.PackageFinding

	for i, component := range pkg.Components {
		arch := config.GetArch(pkg.Metadata.Architecture)
		if !composer.CompatibleComponent(component, arch, flavor) {
			continue
		}

		chain, err := composer.NewImportChain(ctx, component, i, pkg.Metadata.Name, arch, flavor)
		if err != nil {
			return nil, err
		}

		node := chain.Head()
		for node != nil {
			component := node.ZarfComponent
			compFindings := fillComponentTemplate(&component, setVariables)
			compFindings = append(compFindings, checkComponent(component, node.Index())...)
			for i := range compFindings {
				compFindings[i].PackagePathOverride = node.ImportLocation()
				compFindings[i].PackageNameOverride = node.OriginalPackageName()
			}
			findings = append(findings, compFindings...)
			node = node.Next()
		}
	}
	return findings, nil
}

func fillComponentTemplate(c *types.ZarfComponent, setVariables map[string]string) []types.PackageFinding {
	var findings []types.PackageFinding
	err := creator.ReloadComponentTemplate(c)
	if err != nil {
		findings = append(findings, types.PackageFinding{
			Description: err.Error(),
			Severity:    types.SevWarn,
		})
	}
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) {
		yamlTemplates, err := utils.FindYamlTemplates(c, templatePrefix, "###")
		if err != nil {
			findings = append(findings, types.PackageFinding{
				Description: err.Error(),
				Severity:    types.SevWarn,
			})
		}

		for key := range yamlTemplates {
			if deprecated {
				findings = append(findings, types.PackageFinding{
					Description: fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					Severity:    types.SevWarn,
				})
			}
			_, present := setVariables[key]
			if !present {
				findings = append(findings, types.PackageFinding{
					Description: lang.UnsetVarLintWarning,
					Severity:    types.SevWarn,
				})
			}
		}
		for key, value := range setVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
	}

	setVarsAndWarn(types.ZarfPackageTemplatePrefix, false)

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	setVarsAndWarn(types.ZarfPackageVariablePrefix, true)

	//nolint: errcheck // This error should bubble up
	utils.ReloadYamlTemplate(c, templateMap)
	return findings
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

// checkComponent runs lint rules against a component
func checkComponent(c types.ZarfComponent, i int) []types.PackageFinding {
	var findings []types.PackageFinding
	findings = append(findings, checkForUnpinnedRepos(c, i)...)
	findings = append(findings, checkForUnpinnedImages(c, i)...)
	findings = append(findings, checkForUnpinnedFiles(c, i)...)
	return findings
}

func checkForUnpinnedRepos(c types.ZarfComponent, i int) []types.PackageFinding {
	var findings []types.PackageFinding
	for j, repo := range c.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", i, j)
		if !isPinnedRepo(repo) {
			findings = append(findings, types.PackageFinding{
				YqPath:      repoYqPath,
				Description: "Unpinned repository",
				Item:        repo,
				Severity:    types.SevWarn,
			})
		}
	}
	return findings
}

func checkForUnpinnedImages(c types.ZarfComponent, i int) []types.PackageFinding {
	var findings []types.PackageFinding
	for j, image := range c.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", i, j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			findings = append(findings, types.PackageFinding{
				YqPath:      imageYqPath,
				Description: "Failed to parse image reference",
				Item:        image,
				Severity:    types.SevWarn,
			})
			continue
		}
		if !pinnedImage {
			findings = append(findings, types.PackageFinding{
				YqPath:      imageYqPath,
				Description: "Image not pinned with digest",
				Item:        image,
				Severity:    types.SevWarn,
			})
		}
	}
	return findings
}

func checkForUnpinnedFiles(c types.ZarfComponent, i int) []types.PackageFinding {
	var findings []types.PackageFinding
	for j, file := range c.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", i, j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			findings = append(findings, types.PackageFinding{
				YqPath:      fileYqPath,
				Description: "No shasum for remote file",
				Item:        file.Source,
				Severity:    types.SevWarn,
			})
		}
	}
	return findings
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

func validateSchema(jsonSchema []byte, untypedZarfPackage interface{}) ([]types.PackageFinding, error) {
	var findings []types.PackageFinding

	schemaErrors, err := runSchema(jsonSchema, untypedZarfPackage)
	if err != nil {
		return nil, err
	}

	if len(schemaErrors) != 0 {
		for _, schemaErr := range schemaErrors {
			findings = append(findings, types.PackageFinding{
				YqPath:      makeFieldPathYqCompat(schemaErr.Field()),
				Description: schemaErr.Description(),
				Severity:    types.SevErr,
			})
		}
	}

	return findings, err
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
