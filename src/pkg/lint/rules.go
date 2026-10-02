// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func isPinnedImage(image string) (bool, error) {
	transformedImage, err := transform.ParseImageRef(image)
	if err != nil {
		if strings.Contains(image, v1alpha1.ZarfPackageTemplatePrefix) ||
			strings.Contains(image, v1alpha1.ZarfPackageVariablePrefix) {
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
	return strings.Contains(repo, "@")
}

// isTemplatedImage returns true if the image reference contains a Zarf template
// or variable placeholder that has not yet been substituted.
func isTemplatedImage(image string) bool {
	return strings.Contains(image, v1alpha1.ZarfPackageTemplatePrefix) ||
		strings.Contains(image, v1alpha1.ZarfPackageVariablePrefix)
}

// imageDomain returns the registry domain explicitly specified in the image
// reference. An empty string is returned when the reference does not include a
// domain, in which case the registry would default to docker.io. This mirrors
// the domain detection used by the distribution/reference library.
func imageDomain(image string) string {
	image = strings.TrimPrefix(image, helpers.OCIURLPrefix)
	i := strings.IndexRune(image, '/')
	if i == -1 {
		return ""
	}
	prefix := image[:i]
	if strings.ContainsAny(prefix, ".:") || prefix == "localhost" || strings.ToLower(prefix) != prefix {
		return prefix
	}
	return ""
}

// hasInternalDomain returns true if the image's registry domain uses the
// reserved .internal top-level domain, which never resolves on the public
// internet and is the recommended convention for locally-built images.
func hasInternalDomain(image string) bool {
	domain := imageDomain(image)
	// Strip any port so domains such as zarf.internal:5000 are still matched.
	if host, _, ok := strings.Cut(domain, ":"); ok {
		domain = host
	}
	return strings.HasSuffix(domain, ".internal")
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

// CheckComponentValuesV1Beta1 checks recommended practices on a resolved v1beta1 component.
// path is the component's location in the source definition.
func CheckComponentValuesV1Beta1(c v1beta1.ComponentSpec, path string) []PackageFinding {
	var findings []PackageFinding
	for i, repo := range c.Repositories {
		if repo.Ref == nil || repo.Ref.Tag == "" && repo.Ref.Branch == "" && repo.Ref.Commit == "" {
			findings = append(findings, PackageFinding{
				YqPath:      fmt.Sprintf("%s.repositories.[%d]", path, i),
				Description: "Unpinned repository",
				Item:        repo.URL,
				Severity:    SevWarn,
			})
		}
	}
	for i, image := range c.Images {
		imagePath := fmt.Sprintf("%s.images.[%d]", path, i)
		pinned, err := isPinnedImage(image.Name)
		switch {
		case err != nil:
			findings = append(findings, PackageFinding{YqPath: imagePath, Description: "Failed to parse image reference", Item: image.Name, Severity: SevWarn})
		case !pinned:
			findings = append(findings, PackageFinding{YqPath: imagePath, Description: "Image not pinned with digest", Item: image.Name, Severity: SevWarn})
		}
		if !isTemplatedImage(image.Name) && imageDomain(image.Name) == "" {
			findings = append(findings, PackageFinding{YqPath: imagePath, Description: "Image reference does not specify a registry domain", Item: image.Name, Severity: SevWarn})
		}
	}
	for i, file := range c.Files {
		if file.Checksum == "" && helpers.IsURL(file.Source) {
			findings = append(findings, PackageFinding{
				YqPath:      fmt.Sprintf("%s.files.[%d]", path, i),
				Description: "No shasum for remote file",
				Item:        file.Source,
				Severity:    SevWarn,
			})
		}
	}
	for i, archive := range c.ImageArchives {
		for j, image := range archive.Images {
			if !isTemplatedImage(image) && !hasInternalDomain(image) {
				findings = append(findings, PackageFinding{
					YqPath:      fmt.Sprintf("%s.imageArchives.[%d].images.[%d]", path, i, j),
					Description: "Image archive image should use a .internal domain to avoid resolving to a public registry",
					Item:        image,
					Severity:    SevWarn,
				})
			}
		}
	}
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
