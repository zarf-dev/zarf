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

func (ic *ImportChain) LintChain() []lint.ValidatorMessage {
	findings := []lint.ValidatorMessage{}
	baseComponent := ic.Head()

	node := baseComponent
	for node != nil {
		findings = append(findings, node.checkForVarInComponentImport()...)
		findings = append(findings, node.fillComponentTemplate()...)
		findings = append(findings, node.lint()...)
		node = node.Next()
	}
	return findings
}

func (node *Node) fillComponentTemplate() []lint.ValidatorMessage {
	templateMap := map[string]string{}
	findings := []lint.ValidatorMessage{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) {
		yamlTemplates, err := utils.FindYamlTemplates(node, templatePrefix, "###")
		if err != nil {
			findings = append(findings, lint.ValidatorMessage{
				Description:    err.Error(),
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Category:       lint.CategoryWarning,
			})
		}

		for key := range yamlTemplates {
			if deprecated {
				findings = append(findings, lint.ValidatorMessage{
					Description:    fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					PackageRelPath: node.ImportLocation(),
					PackageName:    node.OriginalPackageName(),
					Category:       lint.CategoryWarning,
				})
			}
			// TODO
			// _, present := createOpts.SetVariables[key]
			// if !present {
			// 	validator.addWarning(lint.ValidatorMessage{
			// 		Description:    lang.UnsetVarLintWarning,
			// 		PackageRelPath: node.ImportLocation(),
			// 		PackageName:    node.OriginalPackageName(),
			// 	})
			// }
		}
		// for key, value := range createOpts.SetVariables {
		// 	templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		// }
	}

	setVarsAndWarn(types.ZarfPackageTemplatePrefix, false)

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	setVarsAndWarn(types.ZarfPackageVariablePrefix, true)

	utils.ReloadYamlTemplate(node, templateMap)
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

func (node *Node) lint() []lint.ValidatorMessage {
	findings := node.checkForUnpinnedRepos()
	findings = append(findings, node.checkForUnpinnedImages()...)
	findings = append(findings, node.checkForUnpinnedFiles()...)
	return findings
}

func (node *Node) checkForUnpinnedRepos() []lint.ValidatorMessage {
	findings := []lint.ValidatorMessage{}
	for j, repo := range node.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", node.Index(), j)
		if !isPinnedRepo(repo) {
			findings = append(findings, lint.ValidatorMessage{
				YqPath:         repoYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "Unpinned repository",
				Item:           repo,
				Category:       lint.CategoryWarning,
			})
		}
	}
	return findings
}

func (node *Node) checkForUnpinnedImages() []lint.ValidatorMessage {
	findings := []lint.ValidatorMessage{}
	for j, image := range node.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", node.Index(), j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			findings = append(findings, lint.ValidatorMessage{
				YqPath:         imageYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "Invalid image reference",
				Item:           image,
				Category:       lint.CategoryError,
			})
			continue
		}
		if !pinnedImage {
			findings = append(findings, lint.ValidatorMessage{
				YqPath:         imageYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "Image not pinned with digest",
				Item:           image,
				Category:       lint.CategoryWarning,
			})
		}
	}
	return findings
}

func (node *Node) checkForUnpinnedFiles() []lint.ValidatorMessage {
	findings := []lint.ValidatorMessage{}
	for j, file := range node.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", node.Index(), j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			findings = append(findings, lint.ValidatorMessage{
				YqPath:         fileYqPath,
				PackageRelPath: node.ImportLocation(),
				PackageName:    node.OriginalPackageName(),
				Description:    "No shasum for remote file",
				Item:           file.Source,
				Category:       lint.CategoryWarning,
			})
		}
	}
	return findings
}

func (node *Node) checkForVarInComponentImport() []lint.ValidatorMessage {
	findings := []lint.ValidatorMessage{}
	if strings.Contains(node.Import.Path, types.ZarfPackageTemplatePrefix) {
		findings = append(findings, lint.ValidatorMessage{
			YqPath:         fmt.Sprintf(".components.[%d].import.path", node.Index()),
			PackageRelPath: node.ImportLocation(),
			PackageName:    node.OriginalPackageName(),
			Description:    "Zarf does not evaluate variables at component.x.import.path",
			Item:           node.Import.Path,
			Category:       lint.CategoryWarning,
		})
	}
	if strings.Contains(node.Import.URL, types.ZarfPackageTemplatePrefix) {
		findings = append(findings, lint.ValidatorMessage{
			YqPath:         fmt.Sprintf(".components.[%d].import.url", node.Index()),
			PackageRelPath: node.ImportLocation(),
			PackageName:    node.OriginalPackageName(),
			Description:    "Zarf does not evaluate variables at component.x.import.url",
			Item:           node.Import.URL,
			Category:       lint.CategoryWarning,
		})
	}
	return findings
}
