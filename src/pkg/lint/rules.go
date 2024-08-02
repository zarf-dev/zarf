// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/types"
)

func isPinnedImage(image string) (bool, error) {
	transformedImage, err := transform.ParseImageRef(image)
	if err != nil {
		if strings.Contains(image, types.ZarfPackageTemplatePrefix) ||
			strings.Contains(image, types.ZarfPackageVariablePrefix) {
			return true, nil
		}
		return false, err
	}
	if isCosignSignature(transformedImage.Tag) || isCosignAttestation(transformedImage.Tag) {
		return true, nil
	}
	return (transformedImage.Digest != ""), err
}

func isCosignSignature(image string) bool {
	return strings.HasSuffix(image, ".sig")
}

func isCosignAttestation(image string) bool {
	return strings.HasSuffix(image, ".att")
}

func isPinnedRepo(repo string) bool {
	return (strings.Contains(repo, "@"))
}

// CheckComponentValues runs lint rules validating values on component keys, should be run after templating
func CheckComponentValues(c types.ZarfComponent, i int) []PackageFinding {
	var findings []PackageFinding
	findings = append(findings, checkForUnpinnedRepos(c, i)...)
	findings = append(findings, checkForUnpinnedImages(c, i)...)
	findings = append(findings, checkForUnpinnedFiles(c, i)...)
	return findings
}

func checkForUnpinnedRepos(c types.ZarfComponent, i int) []PackageFinding {
	var findings []PackageFinding
	for j, repo := range c.Repos {
		repoYqPath := fmt.Sprintf(".components.[%d].repos.[%d]", i, j)
		if !isPinnedRepo(repo) {
			findings = append(findings, PackageFinding{
				YqPath:      repoYqPath,
				Description: "Unpinned repository",
				Item:        repo,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}

func checkForUnpinnedImages(c types.ZarfComponent, i int) []PackageFinding {
	var findings []PackageFinding
	for j, image := range c.Images {
		imageYqPath := fmt.Sprintf(".components.[%d].images.[%d]", i, j)
		pinnedImage, err := isPinnedImage(image)
		if err != nil {
			findings = append(findings, PackageFinding{
				YqPath:      imageYqPath,
				Description: "Failed to parse image reference",
				Item:        image,
				Severity:    SevWarn,
			})
			continue
		}
		if !pinnedImage {
			findings = append(findings, PackageFinding{
				YqPath:      imageYqPath,
				Description: "Image not pinned with digest",
				Item:        image,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}

func checkForUnpinnedFiles(c types.ZarfComponent, i int) []PackageFinding {
	var findings []PackageFinding
	for j, file := range c.Files {
		fileYqPath := fmt.Sprintf(".components.[%d].files.[%d]", i, j)
		if file.Shasum == "" && helpers.IsURL(file.Source) {
			findings = append(findings, PackageFinding{
				YqPath:      fileYqPath,
				Description: "No shasum for remote file",
				Item:        file.Source,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}
