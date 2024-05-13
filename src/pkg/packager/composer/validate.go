// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf Packages.
package composer

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/packager/lint"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

func (ic *ImportChain) lintChain() []lint.ValidatorMessage {
	findings := []lint.ValidatorMessage{}
	baseComponent := ic.Head()

	node := baseComponent
	for node != nil {
		node.checkForVarInComponentImport(findings)
		node.fillComponentTemplate(findings)
		node.lint(findings)
		node = node.Next()
	}
	return findings
}

func (node *Node) fillComponentTemplate(findings []lint.ValidatorMessage) []lint.ValidatorMessage {
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) {
		yamlTemplates, err := utils.FindYamlTemplates(node, templatePrefix, "###")
		if err != nil {
			validator.addWarning(lint.ValidatorMessage{
				Description:    err.Error(),
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
			})
		}

		for key := range yamlTemplates {
			if deprecated {
				validator.addWarning(lint.ValidatorMessage{
					Description:    fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					PackageRelPath: node.ImportLocation(),
					PackageName:    node.OriginalPackageName(),
				})
			}
			_, present := createOpts.SetVariables[key]
			if !present {
				validator.addWarning(lint.ValidatorMessage{
					Description:    lang.UnsetVarLintWarning,
					PackageRelPath: node.ImportLocation(),
					PackageName:    node.OriginalPackageName(),
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

func (node *Node) lint(findings []lint.ValidatorMessage) []lint.ValidatorMessage {
	findings = append(findings, node.checkForUnpinnedRepos(findings)...)
	findings = append(findings, node.checkForUnpinnedImages(findings)...)
	findings = append(findings, node.checkForUnpinnedFiles(findings)...)
	return findings
}

func (node *Node) checkForUnpinnedRepos(findings []lint.ValidatorMessage) []lint.ValidatorMessage {
	for j, repo := range node.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", node.Index(), j)
		if !isPinnedRepo(repo) {
			validator.addWarning(lint.ValidatorMessage{
				yqPath:         repoYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "Unpinned repository",
				item:           repo,
			})
		}
	}
}

func (node *Node) checkForUnpinnedImages(findings []lint.ValidatorMessage) []lint.ValidatorMessage {
	for j, image := range node.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", node.Index(), j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			validator.addError(lint.ValidatorMessage{
				yqPath:         imageYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "Invalid image reference",
				item:           image,
			})
			continue
		}
		if !pinnedImage {
			validator.addWarning(lint.ValidatorMessage{
				yqPath:         imageYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "Image not pinned with digest",
				item:           image,
			})
		}
	}
}

func (node *Node) checkForUnpinnedFiles(findings []lint.ValidatorMessage) []lint.ValidatorMessage {
	for j, file := range node.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", node.Index(), j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			validator.addWarning(lint.ValidatorMessage{
				yqPath:         fileYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "No shasum for remote file",
				item:           file.Source,
			})
		}
	}
}

func (node *Node) checkForVarInComponentImport(findings []lint.ValidatorMessage) []lint.ValidatorMessage {
	if strings.Contains(node.Import.Path, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(lint.ValidatorMessage{
			yqPath:         fmt.Sprintf(".components.[%d].import.path", node.Index()),
			PackageRelPath: node.ImportLocation(),
			PackageName:    node.OriginalPackageName(),
			Description:    "Zarf does not evaluate variables at component.x.import.path",
			item:           node.Import.Path,
		})
	}
	if strings.Contains(node.Import.URL, types.ZarfPackageTemplatePrefix) {
		validator.addWarning(lint.ValidatorMessage{
			yqPath:         fmt.Sprintf(".components.[%d].import.url", node.Index()),
			PackageRelPath: node.ImportLocation(),
			PackageName:    node.OriginalPackageName(),
			Description:    "Zarf does not evaluate variables at component.x.import.url",
			item:           node.Import.URL,
		})
	}
}
