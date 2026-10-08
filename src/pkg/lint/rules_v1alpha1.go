// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package lint

import (
	"fmt"
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
)

func isPinnedImage(image string) (bool, error) {
	pinned, err := isPinnedImageReference(image)
	if err != nil {
		if strings.Contains(image, v1alpha1.ZarfPackageTemplatePrefix) ||
			strings.Contains(image, v1alpha1.ZarfPackageVariablePrefix) {
			return true, nil
		}
	}
	return pinned, err
}

func isPinnedRepo(repo string) bool {
	return strings.Contains(repo, "@")
}

// isTemplatedImage returns true if the image reference contains a Zarf template
// or variable placeholder that has not yet been substituted.
func isTemplatedImage(image string) bool {
	return strings.Contains(image, v1alpha1.ZarfPackageTemplatePrefix) ||
		strings.Contains(image, v1alpha1.ZarfPackageVariablePrefix)
}

// CheckComponentValues runs lint rules validating values on component keys, should be run after templating
func CheckComponentValues(c v1alpha1.ZarfComponent, i int) []PackageFinding {
	var findings []PackageFinding
	findings = append(findings, checkForUnpinnedRepos(c, i)...)
	findings = append(findings, checkForUnpinnedImages(c, i)...)
	findings = append(findings, checkForUnpinnedFiles(c, i)...)
	findings = append(findings, checkForImagesWithoutDomain(c, i)...)
	findings = append(findings, checkForImageArchivesWithoutInternalDomain(c, i)...)
	return findings
}

func checkForUnpinnedRepos(c v1alpha1.ZarfComponent, i int) []PackageFinding {
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

func checkForUnpinnedImages(c v1alpha1.ZarfComponent, i int) []PackageFinding {
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

func checkForImagesWithoutDomain(c v1alpha1.ZarfComponent, i int) []PackageFinding {
	var findings []PackageFinding
	for j, image := range c.Images {
		if isTemplatedImage(image) {
			continue
		}
		if imageDomain(image) == "" {
			findings = append(findings, PackageFinding{
				YqPath:      fmt.Sprintf(".components.[%d].images.[%d]", i, j),
				Description: "Image reference does not specify a registry domain",
				Item:        image,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}

func checkForImageArchivesWithoutInternalDomain(c v1alpha1.ZarfComponent, i int) []PackageFinding {
	var findings []PackageFinding
	for j, archive := range c.ImageArchives {
		for k, image := range archive.Images {
			if isTemplatedImage(image) {
				continue
			}
			if !hasInternalDomain(image) {
				findings = append(findings, PackageFinding{
					YqPath:      fmt.Sprintf(".components.[%d].imageArchives.[%d].images.[%d]", i, j, k),
					Description: "Image archive image should use a .internal domain to avoid resolving to a public registry",
					Item:        image,
					Severity:    SevWarn,
				})
			}
		}
	}
	return findings
}

func checkForUnpinnedFiles(c v1alpha1.ZarfComponent, i int) []PackageFinding {
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
